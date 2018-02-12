package io.enmasse.systemtest.resources;

public class AddressSpaceResource {

    private String name;
    private double min;
    private double max;

    public AddressSpaceResource(String name, double min, double max) {
        this.name = name;
        this.min = min;
        this.max = max;
    }

    public String getName() {
        return name;
    }

    public double getMin() {
        return min;
    }

    public double getMax() {
        return max;
    }

}
